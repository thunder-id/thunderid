// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package backchannel

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	certmodel "github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwemock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testIssuer    = "https://op.example.com"
	testClientID  = "rp-client"
	testSubjectID = "user-1"
	testSessionID = "01a0d89e-fc9a-7049-885d-00ebaa20ab41"
	signedToken   = "signed.logout.token"
)

type LogoutTokenBuilderTestSuite struct {
	suite.Suite
	jwtService *jwtmock.JWTServiceInterfaceMock
	jweService *jwemock.JWEServiceInterfaceMock
}

func TestLogoutTokenBuilderTestSuite(t *testing.T) {
	suite.Run(t, new(LogoutTokenBuilderTestSuite))
}

func (s *LogoutTokenBuilderTestSuite) SetupTest() {
	s.jwtService = jwtmock.NewJWTServiceInterfaceMock(s.T())
	s.jweService = jwemock.NewJWEServiceInterfaceMock(s.T())
}

func (s *LogoutTokenBuilderTestSuite) newBuilder(validity int64) LogoutTokenBuilder {
	cfg := oauthconfig.Config{JWT: engineconfig.JWTConfig{Issuer: testIssuer, ValidityPeriod: 3600}}
	cfg.OAuth.Logout.Backchannel.TokenValidityPeriod = validity
	return NewLogoutTokenBuilder(cfg, s.jwtService, s.jweService, jwksresolver.Initialize(nil))
}

func plainClient() *providers.OAuthClient {
	return &providers.OAuthClient{ClientID: testClientID}
}

// captureClaims expects one logout-typed GenerateJWT call and records the claims it was asked to sign.
func (s *LogoutTokenBuilderTestSuite) captureClaims(sub string, validity int64, alg string) *map[string]interface{} {
	var got map[string]interface{}
	s.jwtService.EXPECT().
		GenerateJWT(mock.Anything, sub, testIssuer, validity, mock.Anything, jwt.TokenTypeLogout, alg).
		Run(func(_ context.Context, _, _ string, _ int64, claims map[string]interface{}, _, _ string) {
			got = claims
		}).Return(signedToken, int64(1700000000), nil).Once()
	return &got
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_ClaimShape() {
	claims := s.captureClaims(tokenservice.SubjectClaim(testSubjectID), 120, "")

	token, err := s.newBuilder(120).Build(context.Background(), plainClient(), testSubjectID, testSessionID)

	s.Require().NoError(err)
	s.Equal(signedToken, token)
	s.Equal(testClientID, (*claims)["aud"])
	s.Equal(testSessionID, (*claims)["sid"])
	events, ok := (*claims)["events"].(map[string]interface{})
	s.Require().True(ok, "events must be an object")
	s.Len(events, 1)
	s.Equal(map[string]interface{}{}, events[eventBackchannelLogout], "the member value is an empty object")
	s.NotContains(*claims, "nonce", "a logout token must never carry nonce")
	s.NotContains(*claims, "sub", "sub is the positional argument, not a claim map entry")
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_DefaultsValidityWhenUnset() {
	s.captureClaims(testSubjectID, defaultTokenValidityPeriod, "")

	_, err := s.newBuilder(0).Build(context.Background(), plainClient(), testSubjectID, testSessionID)

	s.NoError(err)
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_UsesTheClientsIDTokenSigningAlgorithm() {
	client := plainClient()
	client.Token = &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{SigningAlg: "ES256"}}
	s.captureClaims(testSubjectID, 60, "ES256")

	_, err := s.newBuilder(60).Build(context.Background(), client, testSubjectID, testSessionID)

	s.NoError(err)
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_RejectsMissingInputs() {
	b := s.newBuilder(120)
	cases := []struct {
		name      string
		client    *providers.OAuthClient
		subject   string
		sessionID string
	}{
		{"nil client", nil, testSubjectID, testSessionID},
		{"client without id", &providers.OAuthClient{}, testSubjectID, testSessionID},
		{"empty subject", plainClient(), "", testSessionID},
		{"empty session", plainClient(), testSubjectID, ""},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := b.Build(context.Background(), tc.client, tc.subject, tc.sessionID)
			s.Error(err)
		})
	}
	s.jwtService.AssertNotCalled(s.T(), "GenerateJWT")
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_SigningFailurePropagates() {
	s.jwtService.EXPECT().GenerateJWT(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).Return("", 0, &tidcommon.InternalServerError).Once()

	_, err := s.newBuilder(120).Build(context.Background(), plainClient(), testSubjectID, testSessionID)

	s.Error(err)
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_EncryptsForAClientThatEncryptsIDTokensAndReplicatesIss() {
	client := encryptingClient(s.T())
	s.captureClaims(testSubjectID, 120, "")
	var header map[string]interface{}
	s.jweService.EXPECT().Encrypt(mock.Anything, []byte(signedToken), mock.Anything, "RSA-OAEP-256",
		jwe.ContentEncAlgorithm("A256GCM"), "JWT", "rp-kid", mock.Anything).
		Run(func(_ context.Context, _ []byte, key *providers.KeyRef, _ string, _ jwe.ContentEncAlgorithm,
			_, _ string, opts ...jwe.EncryptOption) {
			s.NotNil(key.PublicKeyJWK, "the client's key is resolved from its certificate")
			header = map[string]interface{}{}
			for _, opt := range opts {
				opt(header)
			}
		}).Return("encrypted.logout.token", nil).Once()

	token, err := s.newBuilder(120).Build(context.Background(), client, testSubjectID, testSessionID)

	s.Require().NoError(err)
	s.Equal("encrypted.logout.token", token)
	s.Equal(testIssuer, header["iss"], "iss is replicated in the JWE protected header")
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_EncryptingClientWithoutJWEServiceFails() {
	s.captureClaims(testSubjectID, 120, "")
	cfg := oauthconfig.Config{JWT: engineconfig.JWTConfig{Issuer: testIssuer}}
	b := NewLogoutTokenBuilder(cfg, s.jwtService, nil, nil)

	_, err := b.Build(context.Background(), encryptingClient(s.T()), testSubjectID, testSessionID)

	s.Error(err, "signing only would be rejected by a client that negotiated encryption")
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_EncryptingClientWithoutAKeyFails() {
	client := encryptingClient(s.T())
	client.Certificate = nil
	s.captureClaims(testSubjectID, 120, "")

	_, err := s.newBuilder(120).Build(context.Background(), client, testSubjectID, testSessionID)

	s.Error(err)
	s.jweService.AssertNotCalled(s.T(), "Encrypt")
}

// encryptingClient is a client that negotiated JWE ID tokens with an inline RSA JWKS.
func encryptingClient(t *testing.T) *providers.OAuthClient {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := map[string]interface{}{
		"kty": "RSA",
		"kid": "rp-kid",
		"use": "enc",
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}
	jwks, _ := json.Marshal(map[string]interface{}{"keys": []interface{}{jwk}})
	return &providers.OAuthClient{
		ClientID: testClientID,
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			ResponseType:  providers.IDTokenResponseTypeJWE,
			EncryptionAlg: "RSA-OAEP-256",
			EncryptionEnc: "A256GCM",
		}},
		Certificate: &inboundmodel.Certificate{Type: certmodel.CertificateTypeJWKS, Value: string(jwks)},
	}
}

func (s *LogoutTokenBuilderTestSuite) TestBuild_EncryptionFailurePropagates() {
	s.captureClaims(testSubjectID, 120, "")
	s.jweService.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("", &tidcommon.InternalServerError).Once()

	_, err := s.newBuilder(120).Build(context.Background(), encryptingClient(s.T()), testSubjectID, testSessionID)

	s.Error(err)
}
