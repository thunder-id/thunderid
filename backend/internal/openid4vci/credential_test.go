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

package openid4vci

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/openid4vci/credential"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

type CredentialTestSuite struct {
	suite.Suite
}

func TestCredentialTestSuite(t *testing.T) {
	suite.Run(t, new(CredentialTestSuite))
}

func (s *CredentialTestSuite) TestScopeString() {
	s.Equal("a b", scopeString(map[string]interface{}{"scope": "a b"}))
	s.Equal("a b", scopeString(map[string]interface{}{"scope": []interface{}{"a", "b"}}))
	s.Equal("a", scopeString(map[string]interface{}{"scope": []interface{}{"a", 1}}))
	s.Equal("", scopeString(map[string]interface{}{"scope": 42}))
	s.Equal("", scopeString(map[string]interface{}{}))
}

func (s *CredentialTestSuite) TestDtoToCredentialConfig() {
	validity := 3600
	cfg := dtoToCredentialConfig(credential.CredentialConfigurationDTO{
		Format:          "",
		VCT:             "urn:v",
		Claims:          []credential.ClaimMapping{{Name: "given_name"}, {Name: "family_name"}},
		ValiditySeconds: &validity,
	})
	s.Equal(credential.DefaultCredentialFormat, cfg.Format)
	s.Equal("urn:v", cfg.VCT)
	s.Equal([]string{"given_name", "family_name"}, cfg.SDClaims)
	s.Equal(time.Hour, cfg.Validity)

	cfg = dtoToCredentialConfig(credential.CredentialConfigurationDTO{Format: "custom", VCT: "v"})
	s.Equal("custom", cfg.Format)
	s.Zero(cfg.Validity)
}

func (s *CredentialTestSuite) TestAuthorizedCredentialByConfigID() {
	ctx := context.Background()

	s.Run("Success", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &service{cfg: serviceConfig{}, creds: creds}
		cfg, err := svc.authorizedCredential(ctx, "eudi-pid", nil)
		s.Require().NoError(err)
		s.Equal("v", cfg.VCT)
	})

	s.Run("Unknown", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "missing").
			Return(nil, &tidcommon.ServiceError{Code: "x"})
		svc := &service{cfg: serviceConfig{}, creds: creds}
		_, err := svc.authorizedCredential(ctx, "missing", nil)
		s.ErrorIs(err, ErrUnsupportedCredential)
	})

	s.Run("ScopeEnforcedNotAuthorized", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &service{cfg: serviceConfig{EnforceScope: true}, creds: creds}
		_, err := svc.authorizedCredential(ctx, "eudi-pid", []string{"other"})
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("ScopeEnforcedAuthorized", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &service{cfg: serviceConfig{EnforceScope: true}, creds: creds}
		cfg, err := svc.authorizedCredential(ctx, "eudi-pid", []string{"eudi-pid"})
		s.Require().NoError(err)
		s.Equal("v", cfg.VCT)
	})
}

func (s *CredentialTestSuite) TestAuthorizedCredentialByScope() {
	ctx := context.Background()

	s.Run("MatchesScope", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(ctx).Return([]credential.CredentialConfigurationDTO{
			{Handle: "a", VCT: "va"},
			{Handle: "b", VCT: "vb"},
		}, nil)
		svc := &service{cfg: serviceConfig{}, creds: creds}
		cfg, err := svc.authorizedCredential(ctx, "", []string{"b"})
		s.Require().NoError(err)
		s.Equal("vb", cfg.VCT)
	})

	s.Run("NoScopeMatch", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(ctx).Return([]credential.CredentialConfigurationDTO{
			{Handle: "a", VCT: "va"},
		}, nil)
		svc := &service{cfg: serviceConfig{}, creds: creds}
		_, err := svc.authorizedCredential(ctx, "", []string{"z"})
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("ListError", func() {
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().ListCredentialConfigurations(ctx).Return(nil, &tidcommon.ServiceError{Code: "x"})
		svc := &service{cfg: serviceConfig{}, creds: creds}
		_, err := svc.authorizedCredential(ctx, "", []string{"a"})
		s.ErrorIs(err, ErrIssuance)
	})
}

func (s *CredentialTestSuite) TestIssueCredentialErrors() {
	ctx := context.Background()
	svc := &service{cfg: serviceConfig{BatchSize: 5}}

	s.Run("MissingToken", func() {
		_, err := svc.IssueCredential(ctx, "", nil)
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("VerifyFails", func() {
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, "tok", "", "").Return(&tidcommon.ServiceError{Code: "x"})
		svc := &service{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc}
		_, err := svc.IssueCredential(ctx, "tok", nil)
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("MissingSubject", func() {
		token := makeToken(s.T(), map[string]any{})
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
		svc := &service{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc}
		_, err := svc.IssueCredential(ctx, token, []byte("{}"))
		s.ErrorIs(err, ErrInvalidToken)
	})

	s.Run("BadBody", func() {
		token := makeToken(s.T(), map[string]any{"sub": "u1"})
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
		svc := &service{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc}
		_, err := svc.IssueCredential(ctx, token, []byte("not-json"))
		s.ErrorIs(err, ErrInvalidRequest)
	})

	s.Run("MissingProof", func() {
		token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &service{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc, creds: creds}
		body, _ := json.Marshal(CredentialRequest{CredentialConfigurationID: "eudi-pid"})
		_, err := svc.IssueCredential(ctx, token, body)
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("BatchSizeExceeded", func() {
		token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
		jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
		jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
		creds := newCredentialReaderMock(s.T())
		creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
			Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
		svc := &service{cfg: serviceConfig{BatchSize: 1}, jwtService: jwtSvc, creds: creds}
		body, _ := json.Marshal(CredentialRequest{
			CredentialConfigurationID: "eudi-pid",
			Proofs:                    &Proofs{JWT: []string{"a", "b"}},
		})
		_, err := svc.IssueCredential(ctx, token, body)
		s.ErrorIs(err, ErrInvalidRequest)
	})
}

func (s *CredentialTestSuite) TestIssueCredentialBadTokenPayload() {
	ctx := context.Background()
	token := "e30.!!!.sig"
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
	svc := &service{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc}
	_, err := svc.IssueCredential(ctx, token, []byte("{}"))
	s.ErrorIs(err, ErrInvalidToken)
}

func (s *CredentialTestSuite) TestIssueCredentialUnauthorizedCredential() {
	ctx := context.Background()
	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "missing"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
	creds := newCredentialReaderMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "missing").
		Return(nil, &tidcommon.ServiceError{Code: "x"})
	svc := &service{cfg: serviceConfig{BatchSize: 5}, jwtService: jwtSvc, creds: creds}
	body, _ := json.Marshal(CredentialRequest{CredentialConfigurationID: "missing"})
	_, err := svc.IssueCredential(ctx, token, body)
	s.ErrorIs(err, ErrUnsupportedCredential)
}

func (s *CredentialTestSuite) TestIssueCredentialVerifyProofsError() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
	creds := newCredentialReaderMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
	svc := &service{
		cfg:        serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		store:      store,
		jwtService: jwtSvc,
		creds:      creds,
	}

	holderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofJWT := signProofJWT(s.T(), holderKey, testIssuer, "unknown-nonce", time.Now())
	body, _ := json.Marshal(CredentialRequest{
		CredentialConfigurationID: "eudi-pid",
		Proof:                     Proof{ProofType: "jwt", JWT: proofJWT},
	})
	_, err := svc.IssueCredential(ctx, token, body)
	s.ErrorIs(err, ErrInvalidNonce)
}

func (s *CredentialTestSuite) TestIssueCredentialResolveClaimsError() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	nonce := "n"
	s.Require().NoError(store.SaveNonce(ctx, &nonceRecord{Nonce: nonce, ExpiresAt: time.Now().Add(time.Minute)}))
	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)
	creds := newCredentialReaderMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	userSvc.EXPECT().GetUser(ctx, "u1", false).Return(nil, &tidcommon.ServiceError{Code: "x"})
	svc := &service{
		cfg:         serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		store:       store,
		jwtService:  jwtSvc,
		userService: userSvc,
		creds:       creds,
	}

	holderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofJWT := signProofJWT(s.T(), holderKey, testIssuer, nonce, time.Now())
	body, _ := json.Marshal(CredentialRequest{
		CredentialConfigurationID: "eudi-pid",
		Proof:                     Proof{ProofType: "jwt", JWT: proofJWT},
	})
	_, err := svc.IssueCredential(ctx, token, body)
	s.ErrorIs(err, ErrUserNotFound)
}

func (s *CredentialTestSuite) TestIssueCredentialSignError() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	nonce := "n"
	s.Require().NoError(store.SaveNonce(ctx, &nonceRecord{Nonce: nonce, ExpiresAt: time.Now().Add(time.Minute)}))
	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)

	validity := 3600
	creds := newCredentialReaderMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{
			Handle: "eudi-pid", VCT: "urn:v", Format: credential.DefaultCredentialFormat,
			ValiditySeconds: &validity,
		}, nil)
	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	attrs, _ := json.Marshal(map[string]interface{}{})
	userSvc.EXPECT().GetUser(ctx, "u1", false).Return(&user.User{ID: "u1", Attributes: attrs}, nil)

	provider := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	provider.EXPECT().Sign(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("sign failed"))
	signer := &issuerSigner{cryptoProvider: provider, signAlg: cryptolib.ECDSASHA256, jwsAlg: "ES256"}

	svc := &service{
		cfg:         serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		signer:      signer,
		store:       store,
		jwtService:  jwtSvc,
		userService: userSvc,
		creds:       creds,
	}

	holderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofJWT := signProofJWT(s.T(), holderKey, testIssuer, nonce, time.Now())
	body, _ := json.Marshal(CredentialRequest{
		CredentialConfigurationID: "eudi-pid",
		Proof:                     Proof{ProofType: "jwt", JWT: proofJWT},
	})
	_, err := svc.IssueCredential(ctx, token, body)
	s.ErrorIs(err, ErrIssuance)
}

func (s *CredentialTestSuite) TestIssueCredentialSuccess() {
	ctx := context.Background()

	signer := newTestSigner(s.T())
	store := newStatefulStore(s.T())
	nonce := "the-nonce"
	s.Require().NoError(store.SaveNonce(ctx, &nonceRecord{Nonce: nonce, ExpiresAt: time.Now().Add(time.Minute)}))

	token := makeToken(s.T(), map[string]any{"sub": "u1", "scope": "eudi-pid"})
	jwtSvc := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtSvc.EXPECT().VerifyJWT(ctx, token, "", "").Return(nil)

	creds := newCredentialReaderMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{
			Handle: "eudi-pid", VCT: "urn:v", Format: credential.DefaultCredentialFormat,
			Claims: []credential.ClaimMapping{{Name: "given_name"}},
		}, nil)

	userSvc := usermock.NewUserServiceInterfaceMock(s.T())
	attrs, _ := json.Marshal(map[string]interface{}{"given_name": "Ada"})
	userSvc.EXPECT().GetUser(ctx, "u1", false).Return(&user.User{ID: "u1", Attributes: attrs}, nil)

	svc := &service{
		cfg:         serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: 5},
		signer:      signer,
		store:       store,
		jwtService:  jwtSvc,
		userService: userSvc,
		creds:       creds,
	}

	holderKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofJWT := signProofJWT(s.T(), holderKey, testIssuer, nonce, time.Now())
	body, _ := json.Marshal(CredentialRequest{
		CredentialConfigurationID: "eudi-pid",
		Proof:                     Proof{ProofType: "jwt", JWT: proofJWT},
	})

	resp, err := svc.IssueCredential(ctx, token, body)
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().Len(resp.Credentials, 1)
	s.NotEmpty(resp.Credentials[0].Credential)
}

// newTestSigner builds an issuerSigner backed by a cryptomock that signs with a
// real in-memory ECDSA key, returning DER signatures the signer reshapes to JWS.
func newTestSigner(t *testing.T) *issuerSigner {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	provider := cryptomock.NewRuntimeCryptoProviderMock(t)
	provider.EXPECT().Sign(mock.Anything, mock.Anything, "ES256", mock.Anything).
		RunAndReturn(func(
			_ context.Context, _ kmprovider.KeyRef, _ string, content []byte,
		) ([]byte, error) {
			digest := sha256.Sum256(content)
			return ecdsa.SignASN1(rand.Reader, key, digest[:])
		}).Maybe()
	signer := &issuerSigner{
		cryptoProvider: provider,
		keyRef:         kmprovider.KeyRef{KeyID: "kid"},
		signAlg:        cryptolib.ECDSASHA256,
		jwsAlg:         "ES256",
		kid:            "kid",
		x5c:            []string{base64.StdEncoding.EncodeToString([]byte("cert"))},
	}
	return signer
}
