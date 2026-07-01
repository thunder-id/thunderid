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
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// verifyProofs validates a batch of OpenID4VCI holder proofs of possession and
// returns one confirmation JWK per proof (to bind into each issued credential's
// cnf). Batch issuance binds each unlinkable copy to a distinct holder key but
// reuses a single c_nonce across all proofs, so every proof is validated against
// its own embedded jwk while each distinct nonce is consumed exactly once.
func (s *service) verifyProofs(ctx context.Context, proofs []Proof) ([]map[string]interface{}, error) {
	jwks := make([]map[string]interface{}, 0, len(proofs))
	nonces := make([]string, 0, len(proofs))
	for _, proof := range proofs {
		jwk, nonce, err := s.checkProof(proof)
		if err != nil {
			return nil, err
		}
		jwks = append(jwks, jwk)
		nonces = append(nonces, nonce)
	}

	consumed := make(map[string]bool, len(nonces))
	for _, nonce := range nonces {
		if consumed[nonce] {
			continue
		}
		if err := s.consumeNonce(ctx, nonce); err != nil {
			return nil, err
		}
		consumed[nonce] = true
	}

	return jwks, nil
}

// checkProof validates a single holder proof JWT — proof typ, signature (against
// the embedded jwk), audience (the credential issuer), and iat freshness — and
// returns the holder's confirmation JWK and the proof's c_nonce. The nonce is
// not consumed here; verifyProofs consumes each distinct nonce once.
func (s *service) checkProof(proof Proof) (map[string]interface{}, string, error) {
	if proof.ProofType != "jwt" || proof.JWT == "" {
		return nil, "", fmt.Errorf("%w: proof must be a jwt proof", ErrInvalidProof)
	}

	header, err := jws.DecodeHeader(proof.JWT)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalidProof, err)
	}
	if typ, _ := header["typ"].(string); typ != proofType {
		return nil, "", fmt.Errorf("%w: unexpected proof typ %q", ErrInvalidProof, typ)
	}
	jwk, ok := header["jwk"].(map[string]interface{})
	if !ok || len(jwk) == 0 {
		return nil, "", fmt.Errorf("%w: proof header missing jwk", ErrInvalidProof)
	}

	if err := verifyJWSWithJWK(proof.JWT, jwk); err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalidProof, err)
	}

	payload, err := jwt.DecodeJWTPayload(proof.JWT)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrInvalidProof, err)
	}
	if aud, _ := payload["aud"].(string); aud != s.cfg.CredentialIssuer {
		return nil, "", fmt.Errorf("%w: proof audience mismatch", ErrInvalidProof)
	}
	if err := s.checkProofIat(payload); err != nil {
		return nil, "", err
	}
	nonce, _ := payload["nonce"].(string)
	if nonce == "" {
		return nil, "", fmt.Errorf("%w: proof missing nonce", ErrInvalidNonce)
	}

	return jwk, nonce, nil
}

// checkProofIat rejects proofs whose iat is in the future or older than the
// configured maximum age (replay protection).
func (s *service) checkProofIat(payload map[string]interface{}) error {
	iatRaw, ok := payload["iat"].(float64)
	if !ok {
		return fmt.Errorf("%w: proof missing iat", ErrInvalidProof)
	}
	iat := time.Unix(int64(iatRaw), 0)
	now := time.Now()
	if iat.After(now.Add(time.Minute)) {
		return fmt.Errorf("%w: proof iat is in the future", ErrInvalidProof)
	}
	if s.cfg.ProofMaxAge > 0 && iat.Before(now.Add(-s.cfg.ProofMaxAge)) {
		return fmt.Errorf("%w: proof iat too old", ErrInvalidProof)
	}
	return nil
}

// consumeNonce validates a proof's c_nonce against the live nonce store and
// deletes it so it cannot be replayed.
func (s *service) consumeNonce(ctx context.Context, nonce string) error {
	rec, ok := s.store.GetNonce(ctx, nonce)
	if !ok || rec == nil {
		return fmt.Errorf("%w: unknown c_nonce", ErrInvalidNonce)
	}
	_ = s.store.DeleteNonce(ctx, nonce)
	if time.Now().After(rec.ExpiresAt) {
		return fmt.Errorf("%w: c_nonce expired", ErrInvalidNonce)
	}
	return nil
}

// verifyJWSWithJWK verifies a compact JWS against the public key in jwk. ECDSA
// signatures are passed in JWS P1363 (r||s) form, and EC keys are assembled as
// *ecdsa.PublicKey, both of which cryptolib.Verify requires.
func verifyJWSWithJWK(token string, jwk map[string]interface{}) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWS format")
	}
	header, err := jws.DecodeHeader(token)
	if err != nil {
		return err
	}
	algStr, _ := header["alg"].(string)
	alg, err := jws.MapAlgorithmToSignAlg(jws.Algorithm(algStr))
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	signingInput := []byte(parts[0] + "." + parts[1])

	switch alg {
	case cryptolib.ECDSASHA256, cryptolib.ECDSASHA384, cryptolib.ECDSASHA512:
		pub, err := ecJWKToECDSAPublicKey(jwk)
		if err != nil {
			return err
		}
		return cryptolib.Verify(signingInput, sig, alg, pub)
	default:
		pub, err := jws.JWKToPublicKey(jwk)
		if err != nil {
			return err
		}
		return cryptolib.Verify(signingInput, sig, alg, pub)
	}
}

// ecJWKToECDSAPublicKey builds an *ecdsa.PublicKey from an EC JWK. cryptolib's
// ECDSA verify path requires *ecdsa.PublicKey (jws.JWKToECPublicKey yields an
// *ecdh key, which it cannot use).
func ecJWKToECDSAPublicKey(jwk map[string]interface{}) (*ecdsa.PublicKey, error) {
	crv, _ := jwk["crv"].(string)
	xStr, _ := jwk["x"].(string)
	yStr, _ := jwk["y"].(string)
	if crv == "" || xStr == "" || yStr == "" {
		return nil, fmt.Errorf("EC JWK missing crv/x/y")
	}

	var curve elliptic.Curve
	var coordLen int
	switch crv {
	case "P-256":
		curve, coordLen = elliptic.P256(), 32
	case "P-384":
		curve, coordLen = elliptic.P384(), 48
	case "P-521":
		curve, coordLen = elliptic.P521(), 66
	default:
		return nil, fmt.Errorf("unsupported EC curve: %s", crv)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(xStr)
	if err != nil {
		return nil, fmt.Errorf("decode EC x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(yStr)
	if err != nil {
		return nil, fmt.Errorf("decode EC y: %w", err)
	}
	if len(xBytes) > coordLen || len(yBytes) > coordLen {
		return nil, fmt.Errorf("EC coordinate exceeds curve size for %s", crv)
	}

	uncompressed := make([]byte, 1+2*coordLen)
	uncompressed[0] = 0x04
	copy(uncompressed[1+coordLen-len(xBytes):1+coordLen], xBytes)
	copy(uncompressed[1+2*coordLen-len(yBytes):], yBytes)

	pub, err := ecdsa.ParseUncompressedPublicKey(curve, uncompressed)
	if err != nil {
		return nil, fmt.Errorf("invalid EC public key: %w", err)
	}
	return pub, nil
}
