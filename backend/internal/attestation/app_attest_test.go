/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package attestation

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	testTeamID   = "ABCDE12345"
	testBundleID = "com.example.myapp"
)

func appleConfig() *providers.AttestationConfig {
	return &providers.AttestationConfig{
		Apple: &providers.AppleAttestationConfig{TeamID: testTeamID, BundleID: testBundleID},
	}
}

// testChain is a synthetic self-signed root + leaf certificate pair standing in for Apple's real
// App Attest root, so the whole chain-verification path can be exercised offline.
type testChain struct {
	rootPool *x509.CertPool
	leafDER  []byte
	leafCert *x509.Certificate
}

// authDataOpts customizes the synthetic authenticator data built by buildAuthData.
type authDataOpts struct {
	rpIDHash     []byte
	flags        byte
	signCount    uint32
	aaguid       [16]byte
	credentialID []byte
	truncateTo   int // if > 0, truncate the final authData to this length
}

func buildAuthData(o authDataOpts) []byte {
	buf := make([]byte, 0, authDataMinLen+len(o.credentialID))
	buf = append(buf, o.rpIDHash...)
	buf = append(buf, o.flags)
	sc := make([]byte, 4)
	binary.BigEndian.PutUint32(sc, o.signCount)
	buf = append(buf, sc...)
	buf = append(buf, o.aaguid[:]...)
	idLen := make([]byte, 2)
	binary.BigEndian.PutUint16(idLen, uint16(len(o.credentialID))) //nolint:gosec // test data, small length
	buf = append(buf, idLen...)
	buf = append(buf, o.credentialID...)
	if o.truncateTo > 0 && o.truncateTo < len(buf) {
		buf = buf[:o.truncateTo]
	}
	return buf
}

func appIDHash(teamID string) []byte {
	sum := sha256.Sum256([]byte(teamID + "." + testBundleID))
	return sum[:]
}

type AppAttestVerifierTestSuite struct {
	suite.Suite
}

func TestAppAttestVerifierTestSuite(t *testing.T) {
	suite.Run(t, new(AppAttestVerifierTestSuite))
}

// newVerifier builds a verifier trusting the given root pool instead of Apple's real root, so tests
// can verify the whole chain offline against a self-signed test root.
func (s *AppAttestVerifierTestSuite) newVerifier(rootPool *x509.CertPool) *appAttestVerifier {
	return &appAttestVerifier{
		rootPool: rootPool,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AppAttestVerifier")),
	}
}

func (s *AppAttestVerifierTestSuite) generateTestChain() *testChain {
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.Require().NoError(err)
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test App Attest Root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	s.Require().NoError(err)
	rootCert, err := x509.ParseCertificate(rootDER)
	s.Require().NoError(err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.Require().NoError(err)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Test credCert"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, rootCert, &leafKey.PublicKey, rootKey)
	s.Require().NoError(err)
	leafCert, err := x509.ParseCertificate(leafDER)
	s.Require().NoError(err)

	rootPool := x509.NewCertPool()
	rootPool.AddCert(rootCert)

	return &testChain{rootPool: rootPool, leafDER: leafDER, leafCert: leafCert}
}

// credentialIDFor computes the expected credential ID for cert's public key, matching
// verifyKeyIdentifier's encoding.
func (s *AppAttestVerifierTestSuite) credentialIDFor(cert *x509.Certificate) []byte {
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	s.Require().True(ok)
	ecdhPub, err := pub.ECDH()
	s.Require().NoError(err)
	sum := sha256.Sum256(ecdhPub.Bytes())
	return sum[:]
}

// buildToken CBOR-encodes and base64-encodes an attestation object for the given chain and
// authData, as presented in the Attestation-Token header.
func (s *AppAttestVerifierTestSuite) buildToken(format string, x5c [][]byte, authData []byte) string {
	obj := attestationObject{Fmt: format}
	obj.AttStmt.X5C = x5c
	obj.AuthData = authData
	raw, err := cbor.Marshal(obj)
	s.Require().NoError(err)
	return base64.StdEncoding.EncodeToString(raw)
}

// assertRejected asserts a definitive rejection: not verified, with no operational error (mapped to
// 401 by the flow layer).
func (s *AppAttestVerifierTestSuite) assertRejected(
	v providers.AttestationProvider, cfg *providers.AttestationConfig, token string) {
	ok, svcErr := v.Verify(context.Background(), cfg, token)
	s.False(ok)
	s.Nil(svcErr)
}

// assertOperationalError asserts an operational failure: not verified, with a service error (mapped
// to 500 by the flow layer).
func (s *AppAttestVerifierTestSuite) assertOperationalError(
	v providers.AttestationProvider, cfg *providers.AttestationConfig, token string) {
	ok, svcErr := v.Verify(context.Background(), cfg, token)
	s.False(ok)
	s.NotNil(svcErr)
}

func (s *AppAttestVerifierTestSuite) TestVerify_Success() {
	chain := s.generateTestChain()
	opts := authDataOpts{
		rpIDHash:     appIDHash(testTeamID),
		flags:        authDataFlagAttestedCD,
		signCount:    0,
		aaguid:       aaguidProduction,
		credentialID: s.credentialIDFor(chain.leafCert),
	}
	token := s.buildToken(appAttestFormat, [][]byte{chain.leafDER}, buildAuthData(opts))

	verifier := s.newVerifier(chain.rootPool)
	ok, svcErr := verifier.Verify(context.Background(), appleConfig(), token)
	s.True(ok)
	s.Nil(svcErr)
}

func (s *AppAttestVerifierTestSuite) TestVerify_DevelopmentAAGUIDAccepted() {
	chain := s.generateTestChain()
	opts := authDataOpts{
		rpIDHash:     appIDHash(testTeamID),
		flags:        authDataFlagAttestedCD,
		signCount:    0,
		aaguid:       aaguidDevelopment,
		credentialID: s.credentialIDFor(chain.leafCert),
	}
	token := s.buildToken(appAttestFormat, [][]byte{chain.leafDER}, buildAuthData(opts))

	verifier := s.newVerifier(chain.rootPool)
	ok, svcErr := verifier.Verify(context.Background(), appleConfig(), token)
	s.True(ok)
	s.Nil(svcErr)
}

// A missing or empty attestation configuration is an operational error, not a token rejection.
func (s *AppAttestVerifierTestSuite) TestVerify_NotConfigured() {
	chain := s.generateTestChain()
	verifier := s.newVerifier(chain.rootPool)

	s.assertOperationalError(verifier, nil, "anything")
	s.assertOperationalError(verifier, &providers.AttestationConfig{}, "anything")
}

// An incomplete configuration (missing Team ID or Bundle ID) is an operational error.
func (s *AppAttestVerifierTestSuite) TestVerify_IncompleteConfig() {
	chain := s.generateTestChain()
	verifier := s.newVerifier(chain.rootPool)

	cases := map[string]*providers.AppleAttestationConfig{
		"missing team id":   {BundleID: testBundleID},
		"missing bundle id": {TeamID: testTeamID},
	}
	for name, apple := range cases {
		s.Run(name, func() {
			s.assertOperationalError(verifier, &providers.AttestationConfig{Apple: apple}, "anything")
		})
	}
}

// A malformed or mismatched token is a definitive rejection, since App Attest is verified offline.
func (s *AppAttestVerifierTestSuite) TestVerify_InvalidPayload() {
	chain := s.generateTestChain()
	verifier := s.newVerifier(chain.rootPool)

	s.Run("not base64", func() {
		s.assertRejected(verifier, appleConfig(), "***not-base64***")
	})

	s.Run("not cbor", func() {
		token := base64.StdEncoding.EncodeToString([]byte("not cbor"))
		s.assertRejected(verifier, appleConfig(), token)
	})

	s.Run("wrong format", func() {
		opts := authDataOpts{
			rpIDHash: appIDHash(testTeamID), flags: authDataFlagAttestedCD,
			aaguid: aaguidProduction, credentialID: s.credentialIDFor(chain.leafCert),
		}
		token := s.buildToken("packed", [][]byte{chain.leafDER}, buildAuthData(opts))
		s.assertRejected(verifier, appleConfig(), token)
	})

	s.Run("missing x5c", func() {
		opts := authDataOpts{
			rpIDHash: appIDHash(testTeamID), flags: authDataFlagAttestedCD,
			aaguid: aaguidProduction, credentialID: s.credentialIDFor(chain.leafCert),
		}
		token := s.buildToken(appAttestFormat, nil, buildAuthData(opts))
		s.assertRejected(verifier, appleConfig(), token)
	})

	s.Run("attested credential data flag not set", func() {
		opts := authDataOpts{
			rpIDHash: appIDHash(testTeamID), flags: 0x00,
			aaguid: aaguidProduction, credentialID: s.credentialIDFor(chain.leafCert),
		}
		token := s.buildToken(appAttestFormat, [][]byte{chain.leafDER}, buildAuthData(opts))
		s.assertRejected(verifier, appleConfig(), token)
	})

	s.Run("truncated authData does not panic", func() {
		opts := authDataOpts{
			rpIDHash: appIDHash(testTeamID), flags: authDataFlagAttestedCD,
			aaguid: aaguidProduction, credentialID: s.credentialIDFor(chain.leafCert),
			truncateTo: 10,
		}
		token := s.buildToken(appAttestFormat, [][]byte{chain.leafDER}, buildAuthData(opts))
		s.NotPanics(func() {
			s.assertRejected(verifier, appleConfig(), token)
		})
	})
}

func (s *AppAttestVerifierTestSuite) TestVerify_CertificateChainInvalid() {
	chain := s.generateTestChain()
	otherChain := s.generateTestChain() // untrusted root, not in verifier's pool

	opts := authDataOpts{
		rpIDHash: appIDHash(testTeamID), flags: authDataFlagAttestedCD,
		aaguid: aaguidProduction, credentialID: s.credentialIDFor(otherChain.leafCert),
	}
	token := s.buildToken(appAttestFormat, [][]byte{otherChain.leafDER}, buildAuthData(opts))

	verifier := s.newVerifier(chain.rootPool)
	s.assertRejected(verifier, appleConfig(), token)
}

func (s *AppAttestVerifierTestSuite) TestVerify_AppIdentifierMismatch() {
	chain := s.generateTestChain()
	opts := authDataOpts{
		rpIDHash: appIDHash("WRONGTEAM"), flags: authDataFlagAttestedCD,
		aaguid: aaguidProduction, credentialID: s.credentialIDFor(chain.leafCert),
	}
	token := s.buildToken(appAttestFormat, [][]byte{chain.leafDER}, buildAuthData(opts))

	verifier := s.newVerifier(chain.rootPool)
	s.assertRejected(verifier, appleConfig(), token)
}

func (s *AppAttestVerifierTestSuite) TestVerify_EnvironmentUnrecognized() {
	chain := s.generateTestChain()
	var bogusAAGUID [16]byte
	copy(bogusAAGUID[:], "not-app-attest!!")

	opts := authDataOpts{
		rpIDHash: appIDHash(testTeamID), flags: authDataFlagAttestedCD,
		aaguid: bogusAAGUID, credentialID: s.credentialIDFor(chain.leafCert),
	}
	token := s.buildToken(appAttestFormat, [][]byte{chain.leafDER}, buildAuthData(opts))

	verifier := s.newVerifier(chain.rootPool)
	s.assertRejected(verifier, appleConfig(), token)
}

func (s *AppAttestVerifierTestSuite) TestVerify_SignCountNonZero() {
	chain := s.generateTestChain()
	opts := authDataOpts{
		rpIDHash: appIDHash(testTeamID), flags: authDataFlagAttestedCD,
		signCount: 1, aaguid: aaguidProduction, credentialID: s.credentialIDFor(chain.leafCert),
	}
	token := s.buildToken(appAttestFormat, [][]byte{chain.leafDER}, buildAuthData(opts))

	verifier := s.newVerifier(chain.rootPool)
	s.assertRejected(verifier, appleConfig(), token)
}

func (s *AppAttestVerifierTestSuite) TestVerify_KeyIdentifierMismatch() {
	chain := s.generateTestChain()
	tamperedCredentialID := make([]byte, 32) // all-zero, does not match the leaf's public key

	opts := authDataOpts{
		rpIDHash: appIDHash(testTeamID), flags: authDataFlagAttestedCD,
		aaguid: aaguidProduction, credentialID: tamperedCredentialID,
	}
	token := s.buildToken(appAttestFormat, [][]byte{chain.leafDER}, buildAuthData(opts))

	verifier := s.newVerifier(chain.rootPool)
	s.assertRejected(verifier, appleConfig(), token)
}

func (s *AppAttestVerifierTestSuite) TestNewAppAttestVerifier_ParsesConfiguredRoot() {
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.Require().NoError(err)
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test App Attest Root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	s.Require().NoError(err)
	rootPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}))

	verifier, err := newAppAttestVerifier(rootPEM)
	s.NoError(err)
	s.NotNil(verifier)
}

func (s *AppAttestVerifierTestSuite) TestNewAppAttestVerifier_MissingRoot() {
	verifier, err := newAppAttestVerifier("")
	s.Error(err)
	s.Nil(verifier)
}

func (s *AppAttestVerifierTestSuite) TestNewAppAttestVerifier_InvalidRoot() {
	verifier, err := newAppAttestVerifier("not a valid pem certificate")
	s.Error(err)
	s.Nil(verifier)
}
