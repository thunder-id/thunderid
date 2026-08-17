// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// certUpdateFirstJWKS and certUpdateSecondJWKS are two distinct inline JWKS documents. The
	// second one replaces the first, which is what drives the certificate update path: the stored
	// certificate record is updated in place rather than recreated.
	certUpdateFirstJWKS = `{"keys":[{"kty":"RSA","use":"sig","kid":"cert-update-key-1",` +
		`"n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_` +
		`BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2Q` +
		`vzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZ` +
		`u0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw","e":"AQAB"}]}`
	certUpdateSecondJWKS = `{"keys":[{"kty":"EC","use":"sig","kid":"cert-update-key-2","crv":"P-256",` +
		`"x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU","y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}]}`

	certUpdateJWKSURI = "https://cert-update.example.com/.well-known/jwks.json"
)

var certUpdateOU = testutils.OrganizationUnit{
	Handle:      "cert-update-test-ou",
	Name:        "Certificate Update Test OU",
	Description: "Organization unit for the application certificate update tests",
	Parent:      nil,
}

// CertificateUpdateTestSuite covers the certificate lifecycle of an OAuth application: the stored
// certificate is created, updated in place, and removed as the application's inbound OAuth profile
// changes across PUT /applications/{id} requests.
type CertificateUpdateTestSuite struct {
	suite.Suite
	ouID string
}

func TestCertificateUpdateTestSuite(t *testing.T) {
	suite.Run(t, new(CertificateUpdateTestSuite))
}

func (ts *CertificateUpdateTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(certUpdateOU)
	ts.Require().NoError(err, "Failed to create the test organization unit")
	ts.ouID = ouID
}

func (ts *CertificateUpdateTestSuite) TearDownSuite() {
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete the test organization unit during teardown: %v", err)
		}
	}
}

// --- helpers ---

// privateKeyJWTApp builds an application whose OAuth profile authenticates with private_key_jwt and
// carries the given certificate.
func (ts *CertificateUpdateTestSuite) privateKeyJWTApp(name, clientID string,
	cert *ApplicationCert) Application {
	return Application{
		OUID:        ts.ouID,
		Name:        name,
		Description: "Application for the certificate update tests",
		URL:         fmt.Sprintf("https://%s.example.com", clientID),
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                clientID,
					RedirectURIs:            []string{fmt.Sprintf("https://%s.example.com/callback", clientID)},
					GrantTypes:              []string{"client_credentials"},
					ResponseTypes:           []string{},
					TokenEndpointAuthMethod: "private_key_jwt",
					Certificate:             cert,
				},
			},
		},
	}
}

// secretApp builds an application whose OAuth profile authenticates with a client secret and
// therefore carries no certificate.
func (ts *CertificateUpdateTestSuite) secretApp(name, clientID, clientSecret string) Application {
	return Application{
		OUID:        ts.ouID,
		Name:        name,
		Description: "Application for the certificate update tests",
		URL:         fmt.Sprintf("https://%s.example.com", clientID),
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                clientID,
					ClientSecret:            clientSecret,
					RedirectURIs:            []string{fmt.Sprintf("https://%s.example.com/callback", clientID)},
					GrantTypes:              []string{"client_credentials"},
					ResponseTypes:           []string{},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}
}

// storedCertificate returns the certificate the server reports for the application's OAuth profile.
func (ts *CertificateUpdateTestSuite) storedCertificate(appID string) *ApplicationCert {
	app, err := getApplicationByID(appID)
	ts.Require().NoError(err, "Failed to read back the application")
	ts.Require().Len(app.InboundAuthConfig, 1, "The application must expose its OAuth profile")
	ts.Require().NotNil(app.InboundAuthConfig[0].OAuthAppConfig, "The OAuth profile must be populated")
	return app.InboundAuthConfig[0].OAuthAppConfig.Certificate
}

// --- tests ---

// TestCertificateValueUpdatedInPlace asserts that replacing the certificate value of an existing
// OAuth profile updates the stored certificate, and that the new value is what later reads return.
func (ts *CertificateUpdateTestSuite) TestCertificateValueUpdatedInPlace() {
	app := ts.privateKeyJWTApp("Cert Update Value App", "cert_update_value_client",
		&ApplicationCert{Type: "JWKS", Value: certUpdateFirstJWKS})
	appID, err := createApplication(app)
	ts.Require().NoError(err, "Failed to create the application with a certificate")
	defer func() { _ = deleteApplication(appID) }()

	stored := ts.storedCertificate(appID)
	ts.Require().NotNil(stored, "The created application must report its certificate")
	ts.Equal("JWKS", stored.Type, "The stored certificate type must be the one registered")
	ts.Equal(certUpdateFirstJWKS, stored.Value, "The stored certificate value must be the one registered")

	updated := app
	updated.ID = appID
	updated.InboundAuthConfig[0].OAuthAppConfig.Certificate =
		&ApplicationCert{Type: "JWKS", Value: certUpdateSecondJWKS}
	ts.Require().NoError(updateApplication(appID, updated), "Failed to update the certificate value")

	stored = ts.storedCertificate(appID)
	ts.Require().NotNil(stored, "The updated application must still report a certificate")
	ts.Equal("JWKS", stored.Type, "The certificate type is unchanged by a value update")
	ts.Equal(certUpdateSecondJWKS, stored.Value, "The updated certificate value must be returned")
}

// TestCertificateTypeSwitchedOnUpdate asserts that switching an inline JWKS certificate to a JWKS
// URI is applied to the existing certificate record.
func (ts *CertificateUpdateTestSuite) TestCertificateTypeSwitchedOnUpdate() {
	app := ts.privateKeyJWTApp("Cert Update Type App", "cert_update_type_client",
		&ApplicationCert{Type: "JWKS", Value: certUpdateFirstJWKS})
	appID, err := createApplication(app)
	ts.Require().NoError(err, "Failed to create the application with a certificate")
	defer func() { _ = deleteApplication(appID) }()

	updated := app
	updated.ID = appID
	updated.InboundAuthConfig[0].OAuthAppConfig.Certificate =
		&ApplicationCert{Type: "JWKS_URI", Value: certUpdateJWKSURI}
	ts.Require().NoError(updateApplication(appID, updated), "Failed to switch the certificate type")

	stored := ts.storedCertificate(appID)
	ts.Require().NotNil(stored, "The updated application must still report a certificate")
	ts.Equal("JWKS_URI", stored.Type, "The switched certificate type must be returned")
	ts.Equal(certUpdateJWKSURI, stored.Value, "The switched certificate value must be returned")
}

// TestCertificateAddedOnUpdate asserts that an application registered without a certificate gets one
// stored when its profile switches to private_key_jwt.
func (ts *CertificateUpdateTestSuite) TestCertificateAddedOnUpdate() {
	app := ts.secretApp("Cert Update Added App", "cert_update_added_client", "cert_update_added_secret")
	appID, err := createApplication(app)
	ts.Require().NoError(err, "Failed to create the application without a certificate")
	defer func() { _ = deleteApplication(appID) }()

	ts.Nil(ts.storedCertificate(appID), "A client secret application must have no certificate")

	updated := ts.privateKeyJWTApp("Cert Update Added App", "cert_update_added_client",
		&ApplicationCert{Type: "JWKS", Value: certUpdateFirstJWKS})
	updated.ID = appID
	ts.Require().NoError(updateApplication(appID, updated), "Failed to add a certificate on update")

	stored := ts.storedCertificate(appID)
	ts.Require().NotNil(stored, "The updated application must report the added certificate")
	ts.Equal(certUpdateFirstJWKS, stored.Value, "The added certificate value must be returned")
}

// TestCertificateRemovedOnUpdate asserts that moving back to client secret authentication removes the
// stored certificate rather than leaving it behind.
func (ts *CertificateUpdateTestSuite) TestCertificateRemovedOnUpdate() {
	app := ts.privateKeyJWTApp("Cert Update Removed App", "cert_update_removed_client",
		&ApplicationCert{Type: "JWKS", Value: certUpdateFirstJWKS})
	appID, err := createApplication(app)
	ts.Require().NoError(err, "Failed to create the application with a certificate")
	defer func() { _ = deleteApplication(appID) }()

	ts.Require().NotNil(ts.storedCertificate(appID), "The created application must report its certificate")

	updated := ts.secretApp("Cert Update Removed App", "cert_update_removed_client",
		"cert_update_removed_secret")
	updated.ID = appID
	ts.Require().NoError(updateApplication(appID, updated), "Failed to remove the certificate on update")

	ts.Nil(ts.storedCertificate(appID), "The certificate must be removed once the profile no longer has one")
}

// TestInvalidCertificateOnUpdateIsRejected asserts that an invalid certificate is rejected on update
// and that the previously stored certificate survives the rejected request.
func (ts *CertificateUpdateTestSuite) TestInvalidCertificateOnUpdateIsRejected() {
	app := ts.privateKeyJWTApp("Cert Update Invalid App", "cert_update_invalid_client",
		&ApplicationCert{Type: "JWKS", Value: certUpdateFirstJWKS})
	appID, err := createApplication(app)
	ts.Require().NoError(err, "Failed to create the application with a certificate")
	defer func() { _ = deleteApplication(appID) }()

	testCases := []struct {
		name string
		cert *ApplicationCert
	}{
		{name: "unsupported certificate type", cert: &ApplicationCert{Type: "PEM", Value: certUpdateFirstJWKS}},
		{name: "inline JWKS with no value", cert: &ApplicationCert{Type: "JWKS", Value: ""}},
		{name: "JWKS URI that is not a URI", cert: &ApplicationCert{Type: "JWKS_URI", Value: "not-a-uri"}},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			updated := app
			updated.ID = appID
			updated.InboundAuthConfig[0].OAuthAppConfig.Certificate = tc.cert
			err := updateApplication(appID, updated)
			ts.Error(err, "An invalid certificate must be rejected on update")

			stored := ts.storedCertificate(appID)
			ts.Require().NotNil(stored, "A rejected update must leave the stored certificate in place")
			ts.Equal(certUpdateFirstJWKS, stored.Value,
				"A rejected update must not change the stored certificate value")
		})
	}
}
