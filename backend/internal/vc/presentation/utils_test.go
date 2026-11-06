// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

// newTestCertificatePEM mints a self-signed CA certificate valid until notAfter and returns it as PEM.
func newTestCertificatePEM(t *testing.T, commonName string, notAfter time.Time) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             notAfter.Add(-48 * time.Hour),
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: pemBlockTypeCertificate, Bytes: der}))
}

// validTestCertificatePEM mints a certificate valid for the next day.
func validTestCertificatePEM(t *testing.T, commonName string) string {
	t.Helper()
	return newTestCertificatePEM(t, commonName, time.Now().Add(24*time.Hour))
}

func (s *UtilsTestSuite) TestParseTrustedAuthorityCertificate() {
	certificatePEM := validTestCertificatePEM(s.T(), "issuer-ca")

	cert, err := ParseTrustedAuthorityCertificate(certificatePEM)
	s.Require().NoError(err)
	s.Equal("issuer-ca", cert.Subject.CommonName)

	cert, err = ParseTrustedAuthorityCertificate("\n" + certificatePEM + "\n\n")
	s.Require().NoError(err)
	s.NotNil(cert)
}

func (s *UtilsTestSuite) TestParseTrustedAuthorityCertificateRejectsInvalidInput() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.Require().NoError(err)
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	s.Require().NoError(err)
	valid := validTestCertificatePEM(s.T(), "issuer-ca")

	cases := map[string]string{
		"empty":            "",
		"not pem":          "not a certificate",
		"public key pem":   string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})),
		"corrupt der":      string(pem.EncodeToMemory(&pem.Block{Type: pemBlockTypeCertificate, Bytes: []byte("x")})),
		"two blocks":       valid + validTestCertificatePEM(s.T(), "other-ca"),
		"trailing garbage": valid + "garbage",
	}
	for name, input := range cases {
		s.Run(name, func() {
			_, err := ParseTrustedAuthorityCertificate(input)
			s.ErrorIs(err, errInvalidCertificatePEM)
		})
	}
}
