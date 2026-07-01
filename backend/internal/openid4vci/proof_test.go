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
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
)

const testIssuer = "https://issuer.example"

// newStatefulStore returns a openID4VCIStoreInterface mock backed by an in-memory
// map, so tests can seed nonces and observe consumption across a round-trip.
func newStatefulStore(t *testing.T) *openID4VCIStoreInterfaceMock {
	t.Helper()
	m := newOpenID4VCIStoreInterfaceMock(t)
	entries := map[string]*nonceRecord{}
	m.EXPECT().SaveNonce(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rec *nonceRecord) error {
			entries[rec.Nonce] = rec
			return nil
		}).Maybe()
	m.EXPECT().GetNonce(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, nonce string) (*nonceRecord, bool) {
			rec, ok := entries[nonce]
			return rec, ok
		}).Maybe()
	m.EXPECT().DeleteNonce(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, nonce string) error {
			delete(entries, nonce)
			return nil
		}).Maybe()
	return m
}

type ProofTestSuite struct {
	suite.Suite
}

func TestProofTestSuite(t *testing.T) {
	suite.Run(t, new(ProofTestSuite))
}

// signProofJWT builds a signed OpenID4VCI holder proof JWT (typ
// openid4vci-proof+jwt) carrying key's public JWK in the header and the given
// audience/nonce/iat in the payload, signed ES256 in JWS P1363 form.
func signProofJWT(t *testing.T, key *ecdsa.PrivateKey, aud, nonce string, iat time.Time) string {
	t.Helper()
	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(key.PublicKey.X.FillBytes(make([]byte, 32))),
		"y":   base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.FillBytes(make([]byte, 32))),
	}
	header := map[string]interface{}{"alg": "ES256", "typ": proofType, "jwk": jwk}
	payload := map[string]interface{}{"aud": aud, "nonce": nonce, "iat": iat.Unix()}

	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)

	p1363, err := cryptolib.Generate([]byte(signingInput), cryptolib.ECDSASHA256, key)
	if err != nil {
		t.Fatalf("sign proof: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(p1363)
}

func newTestService(t *testing.T, store openID4VCIStoreInterface) *service {
	t.Helper()
	return &service{
		cfg:   serviceConfig{CredentialIssuer: testIssuer, ProofMaxAge: time.Minute, BatchSize: defaultBatchSize},
		store: store,
	}
}

// A batch of proofs (one per holder key) sharing a single c_nonce must yield one
// confirmation JWK per proof, and consume the shared nonce exactly once.
func (s *ProofTestSuite) TestVerifyProofsBatchConsumesSharedNonceOnce() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	nonce := "shared-nonce"
	s.Require().NoError(store.SaveNonce(ctx, &nonceRecord{Nonce: nonce, ExpiresAt: time.Now().Add(time.Minute)}))
	svc := newTestService(s.T(), store)

	proofs := make([]Proof, 0, 3)
	for i := 0; i < 3; i++ {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		s.Require().NoError(err)
		proofs = append(proofs, Proof{
			ProofType: "jwt",
			JWT:       signProofJWT(s.T(), key, testIssuer, nonce, time.Now()),
		})
	}

	jwks, err := svc.verifyProofs(ctx, proofs)
	s.Require().NoError(err)
	s.Require().Len(jwks, 3)
	for i, jwk := range jwks {
		_, ok := jwk["x"].(string)
		s.True(ok, "proof %d: confirmation JWK missing x coordinate", i)
	}
	_, ok := store.GetNonce(ctx, nonce)
	s.False(ok, "shared c_nonce should have been consumed")
}

// An unknown c_nonce is rejected for the whole batch.
func (s *ProofTestSuite) TestVerifyProofsRejectsUnknownNonce() {
	ctx := context.Background()
	svc := newTestService(s.T(), newStatefulStore(s.T()))

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofs := []Proof{{ProofType: "jwt", JWT: signProofJWT(s.T(), key, testIssuer, "never-issued", time.Now())}}

	_, err := svc.verifyProofs(ctx, proofs)
	s.Error(err, "expected error for unknown c_nonce")
}

// encodeJWT assembles a compact JWS from a header and payload, appending sig as
// the (already base64url-encoded) signature segment. Useful for crafting proofs
// whose header/payload exercise specific validation branches.
func encodeJWT(t *testing.T, header, payload map[string]interface{}, sig string) string {
	t.Helper()
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(hb) + "." +
		base64.RawURLEncoding.EncodeToString(pb) + "." + sig
}

func validJWK(key *ecdsa.PrivateKey) map[string]interface{} {
	return map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(key.PublicKey.X.FillBytes(make([]byte, 32))),
		"y":   base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.FillBytes(make([]byte, 32))),
	}
}

func (s *ProofTestSuite) TestCheckProofErrors() {
	svc := newTestService(s.T(), newStatefulStore(s.T()))
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	jwk := validJWK(key)

	s.Run("NotJWTProofType", func() {
		_, _, err := svc.checkProof(Proof{ProofType: "cwt", JWT: "x"})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("EmptyJWT", func() {
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: ""})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("UndecodableHeader", func() {
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: "!!!.!!!.sig"})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("WrongTyp", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256", "typ": "wrong", "jwk": jwk},
			map[string]interface{}{}, "AA")
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("MissingJWK", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256", "typ": proofType},
			map[string]interface{}{}, "AA")
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("BadSignature", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256", "typ": proofType, "jwk": jwk},
			map[string]interface{}{"aud": testIssuer, "nonce": "n", "iat": float64(time.Now().Unix())},
			base64.RawURLEncoding.EncodeToString(make([]byte, 64)))
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("AudienceMismatch", func() {
		jwt := signProofJWT(s.T(), key, "https://other", "n", time.Now())
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("BadIat", func() {
		jwt := signProofJWT(s.T(), key, testIssuer, "n", time.Now().Add(2*time.Minute))
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidProof)
	})

	s.Run("MissingNonce", func() {
		jwt := signProofJWT(s.T(), key, testIssuer, "", time.Now())
		_, _, err := svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
		s.ErrorIs(err, ErrInvalidNonce)
	})
}

func (s *ProofTestSuite) TestCheckProofUndecodablePayload() {
	svc := newTestService(s.T(), newStatefulStore(s.T()))
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	jwk := validJWK(key)
	header := map[string]interface{}{"alg": "ES256", "typ": proofType, "jwk": jwk}
	hb, _ := json.Marshal(header)
	headerSeg := base64.RawURLEncoding.EncodeToString(hb)
	signingInput := headerSeg + "." + "!!!"
	p1363, err := cryptolib.Generate([]byte(signingInput), cryptolib.ECDSASHA256, key)
	s.Require().NoError(err)
	jwt := signingInput + "." + base64.RawURLEncoding.EncodeToString(p1363)

	_, _, err = svc.checkProof(Proof{ProofType: "jwt", JWT: jwt})
	s.ErrorIs(err, ErrInvalidProof)
}

func (s *ProofTestSuite) TestCheckProofIat() {
	svc := newTestService(s.T(), newStatefulStore(s.T()))

	s.Run("MissingIat", func() {
		s.ErrorIs(svc.checkProofIat(map[string]interface{}{}), ErrInvalidProof)
	})

	s.Run("Future", func() {
		payload := map[string]interface{}{"iat": float64(time.Now().Add(2 * time.Minute).Unix())}
		s.ErrorIs(svc.checkProofIat(payload), ErrInvalidProof)
	})

	s.Run("TooOld", func() {
		payload := map[string]interface{}{"iat": float64(time.Now().Add(-2 * time.Minute).Unix())}
		s.ErrorIs(svc.checkProofIat(payload), ErrInvalidProof)
	})

	s.Run("Valid", func() {
		payload := map[string]interface{}{"iat": float64(time.Now().Unix())}
		s.NoError(svc.checkProofIat(payload))
	})
}

func (s *ProofTestSuite) TestConsumeNonceExpired() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	s.Require().NoError(store.SaveNonce(ctx, &nonceRecord{Nonce: "old", ExpiresAt: time.Now().Add(-time.Minute)}))
	svc := newTestService(s.T(), store)

	err := svc.consumeNonce(ctx, "old")
	s.ErrorIs(err, ErrInvalidNonce)
	_, ok := store.GetNonce(ctx, "old")
	s.False(ok, "expired nonce should still be deleted")
}

func (s *ProofTestSuite) TestVerifyProofsConsumeNonceError() {
	ctx := context.Background()
	store := newStatefulStore(s.T())
	svc := newTestService(s.T(), store)

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	proofs := []Proof{{ProofType: "jwt", JWT: signProofJWT(s.T(), key, testIssuer, "unknown", time.Now())}}

	_, err := svc.verifyProofs(ctx, proofs)
	s.ErrorIs(err, ErrInvalidNonce)
}

func (s *ProofTestSuite) TestVerifyJWSWithJWKErrors() {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	jwk := validJWK(key)

	s.Run("BadFormat", func() {
		s.Error(verifyJWSWithJWK("a.b", jwk))
	})

	s.Run("UndecodableHeader", func() {
		s.Error(verifyJWSWithJWK("!!!.payload.sig", jwk))
	})

	s.Run("UnsupportedAlg", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "none"},
			map[string]interface{}{}, "AA")
		s.Error(verifyJWSWithJWK(jwt, jwk))
	})

	s.Run("BadSignatureEncoding", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256"},
			map[string]interface{}{}, "!!!")
		s.Error(verifyJWSWithJWK(jwt, jwk))
	})

	s.Run("ECKeyError", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "ES256"},
			map[string]interface{}{}, base64.RawURLEncoding.EncodeToString(make([]byte, 64)))
		s.Error(verifyJWSWithJWK(jwt, map[string]interface{}{"kty": "EC", "crv": "P-256"}))
	})

	s.Run("NonECKeyError", func() {
		jwt := encodeJWT(s.T(),
			map[string]interface{}{"alg": "RS256"},
			map[string]interface{}{}, base64.RawURLEncoding.EncodeToString(make([]byte, 8)))
		s.Error(verifyJWSWithJWK(jwt, map[string]interface{}{"kty": "RSA"}))
	})
}

func (s *ProofTestSuite) TestECJWKToECDSAPublicKey() {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	x := base64.RawURLEncoding.EncodeToString(key.PublicKey.X.FillBytes(make([]byte, 32)))
	y := base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.FillBytes(make([]byte, 32)))

	s.Run("MissingCoords", func() {
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256"})
		s.Error(err)
	})

	s.Run("UnsupportedCurve", func() {
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-999", "x": x, "y": y})
		s.Error(err)
	})

	s.Run("BadX", func() {
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256", "x": "!!!", "y": y})
		s.Error(err)
	})

	s.Run("BadY", func() {
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256", "x": x, "y": "!!!"})
		s.Error(err)
	})

	s.Run("OversizedCoord", func() {
		big := base64.RawURLEncoding.EncodeToString(make([]byte, 40))
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256", "x": big, "y": y})
		s.Error(err)
	})

	s.Run("InvalidPoint", func() {
		zero := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
		_, err := ecJWKToECDSAPublicKey(map[string]interface{}{"crv": "P-256", "x": zero, "y": zero})
		s.Error(err)
	})

	s.Run("ValidP384", func() {
		k384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		pub, err := ecJWKToECDSAPublicKey(map[string]interface{}{
			"crv": "P-384",
			"x":   base64.RawURLEncoding.EncodeToString(k384.PublicKey.X.FillBytes(make([]byte, 48))),
			"y":   base64.RawURLEncoding.EncodeToString(k384.PublicKey.Y.FillBytes(make([]byte, 48))),
		})
		s.Require().NoError(err)
		s.NotNil(pub)
	})
}

func (s *ProofTestSuite) TestHolderProofsPrefersBatchThenSingle() {
	tests := []struct {
		name string
		req  CredentialRequest
		want int
	}{
		{"batch", CredentialRequest{Proofs: &Proofs{JWT: []string{"a", "b", "c"}}}, 3},
		{"single", CredentialRequest{Proof: Proof{ProofType: "jwt", JWT: "a"}}, 1},
		{"batch over single", CredentialRequest{
			Proof:  Proof{ProofType: "jwt", JWT: "single"},
			Proofs: &Proofs{JWT: []string{"a", "b"}},
		}, 2},
		{"empty", CredentialRequest{}, 0},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.want, len(tc.req.holderProofs()))
		})
	}
}
