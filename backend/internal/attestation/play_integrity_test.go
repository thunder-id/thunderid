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
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	playintegrity "google.golang.org/api/playintegrity/v1"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

const (
	testPackageName = "com.example.app"
	testDigest      = "AA:BB:CC"
	testToken       = "integrity-token"
	testCredentials = "{\"type\":\"service_account\"}"
	testEncrypted   = "encrypted-creds"
)

func androidConfig() *providers.AttestationConfig {
	return &providers.AttestationConfig{
		Android: &providers.AndroidAttestationConfig{
			PackageName:               testPackageName,
			CertificateSha256Digests:  []string{testDigest},
			ServiceAccountCredentials: testEncrypted,
		},
	}
}

func payload(pkg, verdict string, digests []string) *playintegrity.TokenPayloadExternal {
	return &playintegrity.TokenPayloadExternal{
		AppIntegrity: &playintegrity.AppIntegrity{
			PackageName:             pkg,
			AppRecognitionVerdict:   verdict,
			CertificateSha256Digest: digests,
		},
	}
}

type PlayIntegrityVerifierTestSuite struct {
	suite.Suite
	decoder  *integrityTokenDecoderMock
	crypto   *cryptomock.RuntimeCryptoProviderMock
	verifier *playIntegrityVerifier
}

func TestPlayIntegrityVerifierTestSuite(t *testing.T) {
	suite.Run(t, new(PlayIntegrityVerifierTestSuite))
}

func (s *PlayIntegrityVerifierTestSuite) SetupTest() {
	s.decoder = newIntegrityTokenDecoderMock(s.T())
	s.crypto = cryptomock.NewRuntimeCryptoProviderMock(s.T())
	s.verifier = &playIntegrityVerifier{
		decoder:   s.decoder,
		cryptoSvc: s.crypto,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "PlayIntegrityVerifier")),
	}
}

// expectDecrypt configures the crypto mock to decrypt the stored test credentials to their
// plaintext form, as every successful path must decrypt before decoding the token.
func (s *PlayIntegrityVerifierTestSuite) expectDecrypt() {
	s.crypto.EXPECT().Decrypt(context.Background(), mock.Anything, mock.Anything, []byte(testEncrypted)).
		Return([]byte(testCredentials), nil)
}

func (s *PlayIntegrityVerifierTestSuite) TestVerify_Success() {
	s.expectDecrypt()
	s.decoder.EXPECT().Decode(mock.Anything, testCredentials, testPackageName, testToken).
		Return(payload(testPackageName, "PLAY_RECOGNIZED", []string{testDigest}), nil)

	verified, svcErr := s.verifier.Verify(context.Background(), androidConfig(), testToken)
	s.True(verified)
	s.Nil(svcErr)
}

func (s *PlayIntegrityVerifierTestSuite) TestVerify_NotConfigured() {
	verified, svcErr := s.verifier.Verify(context.Background(), nil, testToken)
	s.False(verified)
	s.NotNil(svcErr)

	verified, svcErr = s.verifier.Verify(context.Background(), &providers.AttestationConfig{}, testToken)
	s.False(verified)
	s.NotNil(svcErr)
}

func (s *PlayIntegrityVerifierTestSuite) TestVerify_DecryptError() {
	s.crypto.EXPECT().Decrypt(context.Background(), mock.Anything, mock.Anything, []byte(testEncrypted)).
		Return(nil, errors.New("decrypt failure"))

	verified, svcErr := s.verifier.Verify(context.Background(), androidConfig(), testToken)
	s.False(verified)
	s.NotNil(svcErr)
}

func (s *PlayIntegrityVerifierTestSuite) TestVerify_DecoderError() {
	s.expectDecrypt()
	s.decoder.EXPECT().Decode(mock.Anything, testCredentials, testPackageName, testToken).
		Return(nil, errors.New("google api failure"))

	verified, svcErr := s.verifier.Verify(context.Background(), androidConfig(), testToken)
	s.False(verified)
	s.NotNil(svcErr)
}

func (s *PlayIntegrityVerifierTestSuite) TestVerify_InvalidPayload() {
	s.expectDecrypt()
	s.decoder.EXPECT().Decode(mock.Anything, testCredentials, testPackageName, testToken).
		Return(&playintegrity.TokenPayloadExternal{}, nil)

	verified, svcErr := s.verifier.Verify(context.Background(), androidConfig(), testToken)
	s.False(verified)
	s.Nil(svcErr)
}

func (s *PlayIntegrityVerifierTestSuite) TestVerify_PackageNameMismatch() {
	s.expectDecrypt()
	s.decoder.EXPECT().Decode(mock.Anything, testCredentials, testPackageName, testToken).
		Return(payload("com.attacker.app", "PLAY_RECOGNIZED", []string{testDigest}), nil)

	verified, svcErr := s.verifier.Verify(context.Background(), androidConfig(), testToken)
	s.False(verified)
	s.Nil(svcErr)
}

func (s *PlayIntegrityVerifierTestSuite) TestVerify_SigningIdentityMismatch() {
	s.expectDecrypt()
	s.decoder.EXPECT().Decode(mock.Anything, testCredentials, testPackageName, testToken).
		Return(payload(testPackageName, "PLAY_RECOGNIZED", []string{"ZZ:ZZ:ZZ"}), nil)

	verified, svcErr := s.verifier.Verify(context.Background(), androidConfig(), testToken)
	s.False(verified)
	s.Nil(svcErr)
}

func (s *PlayIntegrityVerifierTestSuite) TestVerify_AppNotPlayRecognized() {
	s.expectDecrypt()
	s.decoder.EXPECT().Decode(mock.Anything, testCredentials, testPackageName, testToken).
		Return(payload(testPackageName, "UNRECOGNIZED_VERSION", []string{testDigest}), nil)

	verified, svcErr := s.verifier.Verify(context.Background(), androidConfig(), testToken)
	s.False(verified)
	s.Nil(svcErr)
}

// TestVerify_IncompleteConfig ensures that a configuration missing the package name, the signing
// certificate digests, or the service account credentials is rejected as incomplete before any
// outbound call is made — a partial configuration cannot establish binary identity, and neither the
// credential decryption nor the Play Integrity API call should be reached.
func (s *PlayIntegrityVerifierTestSuite) TestVerify_IncompleteConfig() {
	cases := map[string]*providers.AndroidAttestationConfig{
		"missing package name": {
			CertificateSha256Digests:  []string{testDigest},
			ServiceAccountCredentials: testEncrypted,
		},
		"missing certificate digests": {
			PackageName:               testPackageName,
			ServiceAccountCredentials: testEncrypted,
		},
		"missing service account credentials": {
			PackageName:              testPackageName,
			CertificateSha256Digests: []string{testDigest},
		},
	}

	for name, android := range cases {
		s.Run(name, func() {
			s.SetupTest()

			cfg := &providers.AttestationConfig{Android: android}
			verified, svcErr := s.verifier.Verify(context.Background(), cfg, testToken)
			s.False(verified)
			s.NotNil(svcErr)
		})
	}
}
