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

// Package dpop implements OAuth 2.0 Demonstrating Proof-of-Possession proof verification.
package dpop

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// VerifierInterface verifies DPoP proofs.
type VerifierInterface interface {
	Verify(ctx context.Context, params VerifyParams) (*ProofResult, error)
}

// verifier is the default VerifierInterface implementation.
type verifier struct {
	jtiStore     jti.JTIStoreInterface
	allowedAlgs  map[string]struct{}
	iatWindow    time.Duration
	leeway       time.Duration
	maxJTILength int
	now          func() time.Time
}

// newVerifier constructs a DPoP proof verifier with the given replay store and policy settings.
func newVerifier(
	jtiStore jti.JTIStoreInterface,
	allowedAlgs []string,
	iatWindow, leeway int,
	maxJTILength int,
) VerifierInterface {
	algSet := make(map[string]struct{}, len(allowedAlgs))
	for _, a := range allowedAlgs {
		algSet[a] = struct{}{}
	}
	return &verifier{
		jtiStore:     jtiStore,
		allowedAlgs:  algSet,
		iatWindow:    time.Duration(iatWindow) * time.Second,
		leeway:       time.Duration(leeway) * time.Second,
		maxJTILength: maxJTILength,
		now:          time.Now,
	}
}

// Verify validates a single DPoP proof. Validation failures wrap ErrInvalidProof;
// replays return ErrReplayedProof; ExpectedJkt mismatch returns ErrJktMismatch.
func (v *verifier) Verify(ctx context.Context, params VerifyParams) (*ProofResult, error) {
	if params.Proof == "" {
		return nil, fmt.Errorf("%w: empty proof", ErrInvalidProof)
	}

	alg, jwk, err := v.validateHeader(params.Proof)
	if err != nil {
		return nil, err
	}

	if err := verifyProofSignature(params.Proof, alg, jwk); err != nil {
		return nil, err
	}

	payload, err := jwt.DecodeJWTPayload(params.Proof)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidProof, err.Error())
	}

	iat, jti, err := v.validatePayloadClaims(payload, params)
	if err != nil {
		return nil, err
	}

	if err := validateATH(payload, params.AccessToken); err != nil {
		return nil, err
	}

	jkt, err := jws.ComputeJKT(jwk)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidProof, err.Error())
	}

	confirmed := false
	if params.ExpectedJkt != "" {
		if subtle.ConstantTimeCompare([]byte(jkt), []byte(params.ExpectedJkt)) != 1 {
			return nil, ErrJktMismatch
		}
		confirmed = true
	}

	expiry := iat.Add(v.iatWindow + 2*v.leeway)
	inserted, err := v.jtiStore.RecordJTI(ctx, jtiNamespace, jti, expiry)
	if err != nil {
		return nil, fmt.Errorf("dpop jti store: %w", err)
	}
	if !inserted {
		return nil, ErrReplayedProof
	}

	return &ProofResult{
		JKT:       jkt,
		JWK:       jwk,
		Alg:       alg,
		Confirmed: confirmed,
	}, nil
}

// validateHeader checks the DPoP JWS header for required typ, allowed alg, and an embedded public jwk.
func (v *verifier) validateHeader(proof string) (string, map[string]any, error) {
	header, err := jws.DecodeHeader(proof)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %s", ErrInvalidProof, err.Error())
	}

	typ, _ := header["typ"].(string)
	if typ != dpopJWTType {
		return "", nil, fmt.Errorf("%w: unexpected typ %q", ErrInvalidProof, typ)
	}

	alg, _ := header["alg"].(string)
	if alg == "" {
		return "", nil, fmt.Errorf("%w: missing alg", ErrInvalidProof)
	}
	if _, ok := v.allowedAlgs[alg]; !ok {
		return "", nil, fmt.Errorf("%w: alg %q not allowed", ErrInvalidProof, alg)
	}

	jwkRaw, ok := header["jwk"]
	if !ok {
		return "", nil, fmt.Errorf("%w: missing jwk header", ErrInvalidProof)
	}
	jwk, ok := jwkRaw.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("%w: jwk header is not a JSON object", ErrInvalidProof)
	}
	if member, found := jws.ContainsPrivateMember(jwk); found {
		return "", nil, fmt.Errorf("%w: jwk contains private-key member %q", ErrInvalidProof, member)
	}

	return alg, jwk, nil
}

// verifyProofSignature verifies the proof's JWS signature using the public key from its jwk header.
func verifyProofSignature(proof, alg string, jwk map[string]any) error {
	signAlg, err := jws.MapAlgorithmToSignAlg(jws.Algorithm(alg))
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidProof, err.Error())
	}
	pubKey, err := jws.JWKToPublicKey(jwk)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidProof, err.Error())
	}
	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		return fmt.Errorf("%w: invalid JWS format", ErrInvalidProof)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("%w: invalid JWS signature encoding: %s", ErrInvalidProof, err.Error())
	}
	if err := cryptolab.Verify([]byte(parts[0]+"."+parts[1]), signature, signAlg, pubKey); err != nil {
		return fmt.Errorf("%w: signature verification failed: %s", ErrInvalidProof, err.Error())
	}
	return nil
}

// validatePayloadClaims checks htm/htu binding, the iat acceptance window, and jti presence/length.
func (v *verifier) validatePayloadClaims(payload map[string]any, params VerifyParams) (time.Time, string, error) {
	htm, _ := payload["htm"].(string)
	if htm == "" || htm != params.HTM {
		return time.Time{}, "", fmt.Errorf("%w: htm mismatch", ErrInvalidProof)
	}

	proofHTU, _ := payload["htu"].(string)
	if proofHTU == "" {
		return time.Time{}, "", fmt.Errorf("%w: missing htu", ErrInvalidProof)
	}
	canonProof, err := syshttp.CanonicalizeURL(proofHTU)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("%w: invalid htu in proof: %s", ErrInvalidProof, err.Error())
	}
	canonExpected, err := syshttp.CanonicalizeURL(params.HTU)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid expected htu: %w", err)
	}
	if canonProof != canonExpected {
		return time.Time{}, "", fmt.Errorf("%w: htu mismatch", ErrInvalidProof)
	}

	iatRaw, ok := payload["iat"]
	if !ok {
		return time.Time{}, "", fmt.Errorf("%w: missing iat", ErrInvalidProof)
	}
	iatSec, ok := sysutils.ToInt64(iatRaw)
	if !ok {
		return time.Time{}, "", fmt.Errorf("%w: invalid iat: unexpected numeric type %T", ErrInvalidProof, iatRaw)
	}
	iat := time.Unix(iatSec, 0)
	now := v.now()
	earliest := now.Add(-v.iatWindow - v.leeway)
	latest := now.Add(v.leeway)
	if iat.Before(earliest) || iat.After(latest) {
		return time.Time{}, "", fmt.Errorf("%w: iat out of acceptance window", ErrInvalidProof)
	}

	jti, _ := payload["jti"].(string)
	if jti == "" {
		return time.Time{}, "", fmt.Errorf("%w: missing jti", ErrInvalidProof)
	}
	if len(jti) > v.maxJTILength {
		return time.Time{}, "", fmt.Errorf("%w: jti exceeds max length", ErrInvalidProof)
	}

	return iat, jti, nil
}

// validateATH verifies the ath claim matches the SHA-256 hash of the access token, when one is bound.
func validateATH(payload map[string]any, accessToken string) error {
	if accessToken == "" {
		return nil
	}
	athClaim, ok := payload["ath"].(string)
	if !ok || athClaim == "" {
		return fmt.Errorf("%w: missing ath", ErrInvalidProof)
	}
	sum := sha256.Sum256([]byte(accessToken))
	expectedAth := base64.RawURLEncoding.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(athClaim), []byte(expectedAth)) != 1 {
		return fmt.Errorf("%w: ath mismatch", ErrInvalidProof)
	}
	return nil
}
